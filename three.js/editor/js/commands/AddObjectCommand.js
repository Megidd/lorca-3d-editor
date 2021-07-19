import { Command } from '../Command.js';
import { ObjectLoader } from '../../../build/three.module.js';

/**
 * @param editor Editor
 * @param object THREE.Object3D
 * @constructor
 */
class AddObjectCommand extends Command {

	constructor( editor, object ) {

		super( editor );

		this.type = 'AddObjectCommand';

		this.object = object;
		if ( object !== undefined ) {

			this.name = `Add Object: ${object.name}`;

		}

	}

	execute() {

		// // To be able to pass from JS to Go
		var mesh = this.object;
		if (mesh != null) {
			var geometry = mesh.geometry;
			if (geometry != null) {
				var attrPos = geometry.attributes['position'];
				if (attrPos != null) {
					var positions = attrPos.array;
					console.log('Positions ==', positions);
					console.log(new Date().toLocaleString())
					vrxBff(positions); // Call Go function
				}
				var bufferIdx = geometry.index;
				if (bufferIdx != null) {
					var indices = bufferIdx.array;
					console.log('Indices ==', indices);
					console.log(new Date().toLocaleString())
					idxBff(indices); // Call Go function
				}
			}
		}

		this.editor.addObject( this.object );
		this.editor.select( this.object );

	}

	undo() {

		this.editor.removeObject( this.object );
		this.editor.deselect();

	}

	toJSON() {

		const output = super.toJSON( this );

		output.object = this.object.toJSON();

		return output;

	}

	fromJSON( json ) {

		super.fromJSON( json );

		this.object = this.editor.objectByUuid( json.object.object.uuid );

		if ( this.object === undefined ) {

			const loader = new ObjectLoader();
			this.object = loader.parse( json.object );

		}

	}

}

export { AddObjectCommand };
